describe('base', () => {
  it('should be interactive', () => {
    cy.visit('/');

    // Open base
    cy.contains('h2', 'test_base').should('exist').click();

    // Activate and deactivate keyboard
    cy.get('[aria-label="Keyboard Disabled"]').should('exist').click();
    cy.get('[aria-label="Keyboard Enabled"]').should('exist').click();

    // Select camera
    cy.get('[aria-label="Select Cameras"]').find('[aria-disabled="false"]').click().type('test_camera{enter}{esc}');
    cy.get('[data-stream-preview="test_camera"').find('video');

    // Confirm that camera component can open stream that is active already
    // Open camera
    cy.contains('h2', 'test_camera').should('exist').click();
    
    // View and hide camera
    cy.get('[aria-label="View Camera: test_camera"]').find('button').should('exist').click();
    cy.get('[aria-label="test_camera stream"').should('exist');
    cy.get('[aria-label="Hide Camera: test_camera"]').find('button').should('exist').click();
    cy.get('[aria-label="test_camera stream"').should('not.be.visible');
  });
});

export {};
